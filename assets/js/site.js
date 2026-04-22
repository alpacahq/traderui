setInterval(function() {
  App.orders.fetch({reset: true});
  App.executions.fetch({reset: true});
  App.refreshSessionStatus();
}, 1000);

var App = new( Backbone.View.extend({
  Models: {},
  Views: {},
  Collections: {},

  events: {
    'click a[data-internal]': function(e) {
      e.preventDefault();
      var href = e.currentTarget.getAttribute('href') || '';
      var fragment = href.indexOf('#') >= 0 ? href.slice(href.indexOf('#') + 1) : href.replace(/^\//, '');
      Backbone.history.navigate(fragment, {trigger: true});
    }
  },

  start: function(options) {
    this.symbols = options.symbols || [];
    this.accounts = options.accounts || [];

    this.orderTicket = new App.Models.OrderTicket({
      session_ids: options.session_ids,
      symbols: this.symbols,
      accounts: this.accounts
    });

    this.securityDefinitionForm = new App.Models.SecurityDefinitionForm({
      session_ids: options.session_ids  
    });

    this.orders = new App.Collections.Orders(options.orders);
    this.executions = new App.Collections.Executions(options.executions);
    this.router = new App.Router();

    this.refreshSessionStatus();

    Backbone.history.start({pushState: false});
  },

  showOrders: function() {
    var orderTicketView = new App.Views.OrderTicket({model: this.orderTicket});
    var ordersView = new App.Views.OrdersView({
      collection: this.orders,
      filter: function(order) { return order.get('security_type') !== 'MLEG'; }
    });

    $("#app").html(orderTicketView.render().el);
    $("#app").append(ordersView.render().el);
    $("#nav-order").addClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-multileg").removeClass("active");
    $("#nav-secdef").removeClass("active");
  },

  showExecutions: function() {
    var orderTicketView = new App.Views.OrderTicket({model: this.orderTicket});
    var executionsView = new App.Views.Executions({collection: this.executions});

    $("#app").html(orderTicketView.render().el);
    $("#app").append(executionsView.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").addClass("active");
    $("#nav-multileg").removeClass("active");
    $("#nav-secdef").removeClass("active");
  },

  showMultileg: function() {
    var multilegTicket = new App.Views.MultilegTicket({model: this.orderTicket});
    var ordersView = new App.Views.OrdersView({
      collection: this.orders,
      filter: function(order) { return order.get('security_type') === 'MLEG'; }
    });

    $("#app").html(multilegTicket.render().el);
    $("#app").append(ordersView.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-multileg").addClass("active");
    $("#nav-secdef").removeClass("active");
  },

  showSecurityDefinitions: function() {
    var secDefReq = new App.Views.SecurityDefinitionRequest({model: this.securityDefinitionForm});
    $("#app").html(secDefReq.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-multileg").removeClass("active");
    $("#nav-secdef").addClass("active");
  },

  refreshSessionStatus: function() {
    $.getJSON('/session-status').done(function(status) {
      var $el = $('#session-status');
      $el.empty();
      _.each(_.keys(status).sort(), function(id) {
        var up = status[id];
        $el.append(
          '<li><span class="dot ' + (up ? 'dot-up' : 'dot-down') + '"></span>'
          + _.escape(id) + ' <span class="text-muted">' + (up ? 'logged on' : 'down') + '</span></li>'
        );
      });
    });
  },

  showOrderDetails: function(id) {
    var order = new App.Models.Order({id: id});
    order.fetch({
      success: function() {
        var orderView = new App.Views.OrderDetails({model: order});
        $("#app").html(orderView.render().el);
      },
      error: function() {
        console.log('Failed to fetch!');
      }
    });
  },
  showExecutionDetails: function(id) {
    var execution = new App.Models.Execution({id: id});
    execution.fetch({
      success: function() {
        var executionView = new App.Views.ExecutionDetails({model: execution});
        $("#app").html(executionView.render().el);
      },
      error: function() {
        console.log('Failed to fetch!');
      }
    });
  }
}))({el: document.body});

App.Router = Backbone.Router.extend({
  routes: {
    "": "index", 
    "orders": "index",
    "executions": "executions",
    "multileg": "multileg",
    "secdefs": "secdefs",
    "orders/:id": "orderDetails",
    "executions/:id": "executionDetails",
  },

  index: function(){
    App.showOrders();
  },

  executions: function() {
    App.showExecutions();
  },

  multileg: function() {
    App.showMultileg();
  },

  secdefs: function() {
    App.showSecurityDefinitions();
  },

  orderDetails: function(id) {
    App.showOrderDetails(id)
  },

  executionDetails: function(id) {
    App.showExecutionDetails(id)
  }
});

App.Models.Order = Backbone.Model.extend({
  urlRoot: "/orders",
});

App.Models.Execution = Backbone.Model.extend({
  urlRoot: "/executions"
});

App.Models.SecurityDefinitionRequest = Backbone.Model.extend({
  urlRoot: "securitydefinitionrequest"
});

App.Models.MultilegOrder = Backbone.Model.extend({
  urlRoot: "/multileg-orders"
});

App.Models.OrderTicket = Backbone.Model.extend({});
App.Models.SecurityDefinitionForm = Backbone.Model.extend({});

App.Collections.Orders = Backbone.Collection.extend({
  url: '/orders',
  comparator: 'id'
});

App.Collections.Executions = Backbone.Collection.extend({
  url: '/executions',
  comparator: 'id'
});

App.Views.ExecutionDetails = Backbone.View.extend({
  template: _.template(`
<div class="panel panel-<%= is_leg ? 'info' : 'default' %>">
  <div class="panel-heading">
    <h4>
      <% if (is_leg) { %>
        <span class="label label-info">Leg Fill</span>
      <% } else { %>
        <span class="label label-success">Order Fill</span>
      <% } %>
      Execution #<%= id %>
    </h4>
  </div>
  <div class="panel-body">
    <dl class="dl-horizontal">
      <dt>Exec ID</dt><dd><%= exec_id || "—" %></dd>
      <dt>Order ID</dt><dd><%= order_id || "—" %></dd>
      <dt>ClOrdID</dt><dd><%= clord_id || "—" %></dd>
      <dt>Symbol</dt><dd><%= symbol %></dd>
      <dt>Security Type</dt><dd><%= App.prettySecurityType(security_type) %></dd>
      <dt>Side</dt><dd><%= App.prettySide(side) %></dd>
      <dt>Quantity</dt><dd><%= quantity %></dd>
      <dt>Price</dt><dd><%= price %></dd>
      <dt>Status</dt><dd><span class="label <%= App.ordStatusClass(ord_status) %>"><%= App.prettyOrdStatus(ord_status) %></span></dd>
      <% if (security_type === "OPT") { %>
      <dt>Put or Call</dt><dd><%= App.prettyPutOrCall(put_or_call) %></dd>
      <dt>Strike Price</dt><dd><%= strike_price %></dd>
      <dt>Maturity</dt><dd><%= maturity_month_year %></dd>
      <% } %>
      <dt>Session</dt><dd><%= session_id %></dd>
    </dl>
  </div>
</div>
<button class="btn btn-info back">Back</button>
`),
  render: function() {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    'click .back': function(e) {
      window.history.back();
    }
  }
});



App.Views.OrderDetails = Backbone.View.extend({
  template: _.template(`
<form class="form-horizontal">
  <div class="form-group">
    <label class="col-sm-2 control-label">ID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">ClOrID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= clord_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">OrderID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= order_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Symbol</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= symbol %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Account</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= account %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Session</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= session_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Side</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= App.prettySide(side) %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">OrdType</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= App.prettyOrdType(ord_type) %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Closed</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= closed %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Open</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= open %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Avg Px</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= avg_px %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Security Type</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= security_type %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Security Desc</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= security_desc %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Maturity Month Year</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= maturity_month_year %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Maturity Day</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= maturity_day %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Put or Call</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= put_or_call %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Strike Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= strike_price %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Status</label>
    <div class="col-sm-10">
      <p class="form-control-static"><span class="label <%= App.ordStatusClass(ord_status) %>"><%= App.prettyOrdStatus(ord_status) %></span></p>
    </div>
  </div>
  <% if (typeof rejection_reason !== 'undefined' && rejection_reason) { %>
  <div class="form-group">
    <label class="col-sm-2 control-label">Rejection Reason</label>
    <div class="col-sm-10">
      <p class="form-control-static text-danger"><strong><%= rejection_reason %></strong></p>
    </div>
  </div>
  <% } %>

  <% if (typeof last_op_rejection !== 'undefined' && last_op_rejection) { %>
  <div class="form-group">
    <label class="col-sm-2 control-label">Last Op Reject</label>
    <div class="col-sm-10">
      <p class="form-control-static text-warning"><strong>&#9888; <%= last_op_rejection %></strong></p>
    </div>
  </div>
  <% } %>

  <% if (legs && legs.length) { %>
  <div class="form-group">
    <label class="col-sm-2 control-label">Legs</label>
    <div class="col-sm-10">
      <table class="table table-condensed">
        <thead><tr>
          <th>#</th><th>Symbol</th><th>CFI</th><th>Side</th><th>Ratio</th>
          <th>Strike</th><th>Maturity</th><th>Pos Effect</th>
        </tr></thead>
        <tbody>
          <% _.each(legs, function(l, i){ %>
          <tr>
            <td><%= i + 1 %></td>
            <td><%= l.leg_symbol %></td>
            <td><%= l.leg_cfi_code %></td>
            <td><%= App.prettySide(l.leg_side) %></td>
            <td><%= l.leg_ratio_qty %></td>
            <td><%= l.leg_strike_price || "" %></td>
            <td><%= l.leg_maturity_date || "" %></td>
            <td><%= l.leg_position_effect %></td>
          </tr>
          <% }); %>
        </tbody>
      </table>
    </div>
  </div>
  <% } %>

  <% if (open == "0") { %>
  <div class="form-group">
    <label class="col-sm-2 control-label">Quantity</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= quantity %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= price %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Stop Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= stop_price %></p>
    </div>
  </div>

  <% } else { %>

  <div class="form-group">
    <label for="quantity" class="col-sm-2 control-label">Quantity</label>
    <div class="col-sm-10">
      <input type="number" class="form-control" id="quantity" placeholder="Quantity" value="<%= quantity %>" required>
    </div>
  </div>
  <div class="form-group">
    <label for="price" class="col-sm-2 control-label">Price</label>
    <div class="col-sm-10">
      <input type="number" step=".01" class="form-control" id="price" placeholder="Price" value="<%= price %>">
    </div>
  </div>
  <div class="form-group">
    <label for="stop_price" class="col-sm-2 control-label">Stop Price</label>
    <div class="col-sm-10">
      <input type="number" step=".01" class="form-control" id="stop_price" placeholder="Stop Price" value="<%= stop_price %>">
    </div>
  </div>
  <% } %>

</form>
<button class="btn btn-danger cancel" <% if(open == "0"){%>disabled<% }%>>Cancel</button>
<button class="btn btn-warning amend" <% if(open == "0"){%>disabled<% }%>>Amend</button>
<button class="btn btn-info back">Back</button>
`),
  render: function() {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    'click .back': function(e) {
      window.history.back();
    },

    'click .cancel': function(e) {
      var attrs = this.model.attributes;
      var url = (attrs.security_type === "MLEG")
        ? "/multileg-orders/" + attrs.id
        : "/orders/" + attrs.id;
      $.ajax({
        type: "DELETE",
        url: url,
        success: function() {
          Backbone.history.navigate("orders", {trigger: true});
        },
        error: function(xhr) {
          alert("Cancel failed: " + xhr.responseText);
        }
      });
    },

    'click .amend': function(e) {
      var attrs = this.model.attributes;
      var data = {
        quantity:   this.$('#quantity').val() || attrs.quantity,
        ord_type:   attrs.ord_type,
        price:      this.$('#price').val() || attrs.price,
        stop_price: this.$('#stop_price').val() || attrs.stop_price
      };

      var url = (attrs.security_type === "MLEG")
        ? "/multileg-orders/" + attrs.id
        : "/orders/" + attrs.id;

      $.ajax({
        type: "PUT",
        url: url,
        contentType: "application/json",
        data: JSON.stringify(data),
        success: function() {
          Backbone.history.navigate("orders", {trigger: true});
        },
        error: function(xhr) {
          alert("Amend failed: " + xhr.responseText);
        }
      });
    }
  },
});

App.Views.ExecutionRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-info btn-xs details">Details</button>
</td>
<td>
  <% if (is_leg) { %>
    <span class="label label-info">Leg</span>
  <% } else { %>
    <span class="label label-success">Order</span>
  <% } %>
</td>
<td><%= is_leg ? "↳ " + symbol : symbol %></td>
<td><%= App.prettySecurityType(security_type) %></td>
<td><%= quantity %></td>
<td><%= App.prettySide(side) %></td>
<td><%= price %></td>
<td><%= security_type === "OPT" ? App.prettyPutOrCall(put_or_call) : "" %></td>
<td><%= strike_price || "" %></td>
<td><%= clord_id || "" %></td>
<td><%= session_id %></td>
`),

  render: function() {
    this.$el.html(this.template(this.model.attributes));
    if (this.model.get('is_leg')) {
      this.$el.css('background-color', '#f0f8ff');
    }
    return this;
  },
  events: {
    "click .details": "details"
  },
  details: function(e) {
    Backbone.history.navigate("executions/" + this.model.get("id"), {trigger: true});
  }
});


App.Views.OrderRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-danger cancel" <% if(open == "0"){%>disabled<% }%>>Cancel</button>
<button class="btn btn-info details">Details</button>
</td>
<td><%= symbol %></td>
<td><%= App.prettySecurityType(security_type) %></td>
<td><%= quantity %></td>
<td><%= account %></td>
<td><%= open %></td>
<td><%= closed %></td>
<td><%= App.prettySide(side) %></td>
<td><%= App.prettyOrdType(ord_type) %></td>
<td><%= price %></td>
<td><%= stop_price %></td>
<td><%= avg_px %></td>
<td><span class="label <%= App.ordStatusClass(ord_status) %>"><%= App.prettyOrdStatus(ord_status) %></span></td>
<td><% if(typeof rejection_reason !== 'undefined' && rejection_reason){ %><span class="text-danger" title="<%= rejection_reason %>"><%= rejection_reason %></span><% } %></td>
<td><% if(typeof last_op_rejection !== 'undefined' && last_op_rejection){ %><span class="text-warning" title="<%= last_op_rejection %>">&#9888; <%= last_op_rejection %></span><% } %></td>
<td><%= session_id %></td>
`),

  render: function() {
    this.$el.html(this.template(this.model.attributes));
    this.$el.removeClass('danger warning success');
    var status = this.model.get('ord_status');
    if (status === '8') {
      this.$el.addClass('danger');
    } else if (status === '4') {
      this.$el.addClass('warning');
    } else if (status === '2') {
      this.$el.addClass('success');
    }
    return this;
  },
  events: {
    "click .cancel": "cancel",
    "click .details": "details"
  },
  cancel: function(e) {
    var attrs = this.model.attributes;
    var url = (attrs.security_type === "MLEG")
      ? "/multileg-orders/" + attrs.id
      : "/orders/" + attrs.id;
    $.ajax({
      type: "DELETE",
      url: url,
      error: function(xhr) {
        alert("Cancel failed: " + xhr.responseText);
      }
    });
  },

  details: function(e) {
    Backbone.history.navigate("orders/" + this.model.get("id"), {trigger: true});
  }
});

App.Views.Executions = Backbone.View.extend({
  initialize: function() {
    this.listenTo(this.collection, 'reset', this.addAll);
  },

  render: function() {
    this.$el.html(`
<table class='table' id='executions'>
  <thead>
    <tr>
      <th></th>
      <th>Type</th>
      <th>Symbol</th>
      <th>Security Type</th>
      <th>Quantity</th>
      <th>Side</th>
      <th>Price</th>
      <th>Put/Call</th>
      <th>Strike</th>
      <th>ClOrdID</th>
      <th>Session</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    this.addAll();

    return this;
  },

  addAll: function() {
    this.$("tbody").empty();

    var byId = function(m) { return m.get('id'); };
    var orderExecs = _.sortBy(this.collection.filter(function(e) { return !e.get('is_leg'); }), byId);
    var legExecs = this.collection.filter(function(e) { return e.get('is_leg'); });
    var legsByClOrdID = _.groupBy(legExecs, function(e) { return e.get('clord_id'); });

    var self = this;
    var parentClOrdIDs = {};
    _.each(orderExecs, function(exec) {
      self.addOne(exec);
      parentClOrdIDs[exec.get('clord_id')] = true;
      var legs = _.sortBy(legsByClOrdID[exec.get('clord_id')] || [], byId);
      _.each(legs, function(leg) { self.addOne(leg); });
    });

    var orphanLegs = _.sortBy(
      _.filter(legExecs, function(e) { return !parentClOrdIDs[e.get('clord_id')]; }),
      byId
    );
    _.each(orphanLegs, function(leg) { self.addOne(leg); });

    return this;
  },

  addOne: function(execution) {
    var row = new App.Views.ExecutionRowView({model: execution});
    this.$("tbody").append(row.render().el);
  }
});

App.Views.OrdersView = Backbone.View.extend({
  initialize: function(options) {
    options = options || {};
    this.filter = options.filter || null;
    this.listenTo(this.collection, 'reset update change', this.addAll);
  },

  visibleOrders: function() {
    if (!this.filter) {
      return this.collection.models;
    }
    return this.collection.filter(this.filter, this);
  },

  render: function() {
    this.$el.html(`
<table class='table table-striped' id='orders'>
  <thead>
    <tr>
      <th></th>
      <th>Symbol</th>
      <th>Security Type</th>
      <th>Quantity</th>
      <th>Account</th>
      <th>Open</th>
      <th>Executed</th>
      <th>Side</th>
      <th>Type</th>
      <th>Limit</th>
      <th>Stop</th>
      <th>AvgPx</th>
      <th>Status</th>
      <th>Rejection Reason</th>
      <th>Op Reject</th>
      <th>Session</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    _.each(this.visibleOrders(), this.addOne, this);
    return this;
  },

  addAll: function() {
    this.$("tbody").empty();
    _.each(this.visibleOrders(), this.addOne, this);
    return this;
  },

  addOne: function(order) {
    var row = new App.Views.OrderRowView({model: order});
    this.$("tbody").append(row.render().el);
  }
});

App.Views.SecurityDefinitionRequest = Backbone.View.extend({
  template: _.template(`
<form class='form-inline'>
  <p>
    <div class='form-group'>
      <label for="security_request_type">Security Request Type</label>
      <select class='form-control' name='security_request_type'>
        <option value="0">Security Identity and Specifications</option>
        <option value="1">Security Identity for the Specifications Provided</option>
        <option value="2">List Security Types</option>
        <option value="3">List Securities</option>
      </select>
    </div>
  </p>
  <p>
    <div class='form-group'>
      <label for='security_type'>SecurityType</label>
      <select class='form-control' name='security_type' id='security_type'>
        <option value='CS'>Common Stock</option>
        <option value='FUT'>Future</option>
        <option value='OPT'>Option</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='symbol'>Symbol</label>
      <input type='text' class='form-control' name='symbol' placeholder='Symbol'>
    </div>
  </p>

  <p>
  <div class='form-group'>
    <label for='session'>Session</label>
    <select class='form-control' name='session'>
      <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
    </select>
  </div>

  <button type='submit' class='btn btn-default'>Submit</button>
  </p>
</form>
  `),

  events: {
    submit: "submit"
  },

  submit: function(e) {
    e.preventDefault();
    var req = new App.Models.SecurityDefinitionRequest();
    req.set({
      session_id:             this.$('select[name=session]').val(),
      security_request_type:  this.$('select[name=security_request_type]').val(),
      security_type:          this.$('select[name=security_type]').val(),
      symbol:                 this.$('input[name=symbol]').val(),
    });
    req.save();
  },

  render: function() {
    this.$el.html(this.template(this.model.attributes));
    return this;
  }
});

App.Views.OrderTicket = Backbone.View.extend({
  template: _.template(`
<form class='form-inline' action='/orders' method='POST' id='order-ticket'>
  <p>
    <div class='form-group'>
      <label for='side'>Side</label>
      <select class='form-control' name='side'>
        <option value='1'>Buy</option>
        <option value='2'>Sell</option>
        <option value='5'>Sell Short</option>
        <option value='6'>Sell Short Exempt</option>
        <option value='8'>Cross</option>
        <option value='9'>Cross Short</option>
        <option value='A'>Cross Short Exempt</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='quantity'>Quantity</label>
      <input type='number' class='form-control' name='quantity' placeholder='Quantity' required>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='security_type'>SecurityType</label>
      <select class='form-control' name='security_type' id='security_type'>
        <option value='CS'>Common Stock</option>
        <option value='FUT'>Future</option>
        <option value='OPT'>Option</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='root_select'>Root</label>
      <select class='form-control' name='root_select' id='root_select'>
        <% var roots = _.uniq(_.pluck(symbols, 'symbol')).sort(); %>
        <% _.each(roots, function(r){ %>
          <option value='<%= r %>'><%= r %></option>
        <% }); %>
      </select>
    </div>

    <div class='form-group'>
      <label for='symbol_select'>Symbol</label>
      <select class='form-control' name='symbol_select' id='symbol_select'>
        <option value='__custom__'>-- Custom Symbol --</option>
      </select>
    </div>

    <div class='form-group' id='custom-symbol-group' style='display:none'>
      <label for='symbol'>Custom Symbol</label>
      <input type='text' class='form-control' name='symbol' placeholder='Symbol'>
    </div>

    <div class='form-group'>
      <label for='security_desc'>Security Desc</label>
      <input type='text' class='form-control' name='security_desc' id='security_desc' placeholder='Security Desc'>
    </div>
  </p>
  <p>
    <div class='form-group'>
      <label for='maturity_month_year'>Maturity Month Year</label>
      <input type='text' class='form-control' name='maturity_month_year' id='maturity_month_year' placeholder='Maturity Month Year' disabled>
    </div>

    <div class='form-group'>
      <label for='maturity_day'>Maturity Day</label>
      <input type='number' class='form-control' name='maturity_day' id='maturity_day' placeholder='Maturity Day' disabled>
    </div>

    <div class='form-group'>
      <label for='put_or_call'>Put or Call</label>
      <select class='form-control' name='put_or_call' id='put_or_call' disabled>
        <option value=1>Call</option>
        <option value=0>Put</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='strike_price'>Strike Price</label>
      <input type='number' step='.01' class='form-control' name='strike_price' id='strike_price' placeholder='Strike Price' disabled>
    </div>
  </p>
  <p>
    <div class='form-group'>
      <label for='ordType'>Type</label>
      <select class='form-control' name='ordType' id="ordType">
        <option value='1'>Market</option>
        <option value='2'>Limit</option>
        <option value='3'>Stop</option>
        <option value='4'>Stop Limit</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='limit'>Limit</label>
      <input type='number' step='.01' class='form-control' id="limit" placeholder='Limit' name='price' disabled>
    </div>

    <div class='form-group'>
      <label for='stop'>Stop</label>
      <input type='number' step='.01' class='form-control' id="stop" placeholder='Stop' name='stopPrice' disabled>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='account'>Account</label>
      <select class='form-control' name='account' id='account'>
        <% if (!accounts || !accounts.length) { %>
          <option value=''></option>
        <% } %>
        <% _.each(accounts || [], function(a){ %>
          <option value='<%= a %>'><%= a %></option>
        <% }); %>
      </select>
    </div>

    <div class='form-group'>
      <label for='tif'>TIF</label>
      <select class='form-control' name='tif'>
        <option value='0'>Day</option>
        <option value='3'>IOC</option>
        <option value='2'>OPG</option>
        <option value='1'>GTC</option>
        <option value='5'>GTX</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='openClose'>OpenClose</label>
      <select class='form-control' name='openClose'>
        <option value=''></option>
        <option value='O'>O</option>
        <option value='C'>C</option>
      </select>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='session'>Session</label>
      <select class='form-control' name='session'>
        <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
      </select>
    </div>
  </p>
  <button type='submit' class='btn btn-default'>Submit</button>
</form>
`),
  render: function() {
    this.$el.html(this.template(this.model.attributes));
    this.populateSymbolSelect();
    return this;
  },

  events: {
    "change #ordType": "updateOrdType",
    "change #security_type": "updateSecurityType",
    "change #root_select": "populateSymbolSelect",
    "change #symbol_select": "updateSymbolSelect",
    submit: "submit"
  },

  populateSymbolSelect: function() {
    var root = this.$('#root_select').val();
    var symbols = _.filter(this.model.get('symbols') || [], function(s) {
      return s.symbol === root;
    });

    var $sel = this.$('#symbol_select');
    $sel.empty();
    _.each(symbols, function(s, i) {
      var $opt = $('<option>')
        .attr('value', 'sym-' + i)
        .attr('data-symbol', s.symbol)
        .attr('data-type', s.type)
        .attr('data-cfi', s.cfi_code || '')
        .attr('data-strike', s.strike_price || '')
        .attr('data-maturity-date', s.maturity_date || '')
        .attr('data-maturity-my', s.maturity_month_year || '')
        .attr('data-desc', s.description || '')
        .text(s.description);
      $sel.append($opt);
    });
    $sel.append('<option value="__custom__">-- Custom Symbol --</option>');
    this.updateSymbolSelect();
  },

  submit: function(e) {
    e.preventDefault();
    var sel = this.$('#symbol_select');
    var selVal = sel.val();
    var symbol;
    if (selVal === '__custom__') {
      symbol = this.$('input[name=symbol]').val();
    } else {
      symbol = sel.find(':selected').data('symbol') || selVal;
    }

    var order = new App.Models.Order();
    order.set({
      side:                 this.$('select[name=side]').val(),
      quantity:             this.$('input[name=quantity]').val(),
      symbol:               symbol,
      ord_type:             this.$('select[name=ordType]').val(),
      price:                this.$('input[name=price]').val(),
      stop_price:           this.$('input[name=stopPrice]').val(),
      account:              this.$('[name=account]').val(),
      tif:                  this.$('select[name=tif]').val(),
      open_close:           this.$('select[name=openClose]').val(),
      session_id:           this.$('select[name=session]').val(),
      security_type:        this.$('select[name=security_type]').val(),
      security_desc:        this.$('#security_desc').val(),
      maturity_month_year:  this.$('input[name=maturity_month_year]').val(),
      maturity_day:         parseInt(this.$('input[name=maturity_day]').val()),
      put_or_call:          this.$('select[name=put_or_call]').val(),
      strike_price:         this.$('input[name=strike_price]').val(),
    });

    var self = this;
    order.save(null, {
      success: function() {
        self.$('input[name=quantity]').val('');
        self.$('input[name=price]').val('');
        self.$('input[name=stopPrice]').val('');
      },
      error: function(model, response) {
        alert("Order rejected: " + response.responseText);
      }
    });
  },

  updateSymbolSelect: function() {
    var sel = this.$('#symbol_select');
    var val = sel.val();
    if (val === '__custom__') {
      this.$('#custom-symbol-group').show();
      this.$('input[name=symbol]').prop('required', true);
      return;
    }
    this.$('#custom-symbol-group').hide();
    this.$('input[name=symbol]').prop('required', false);

    var opt = sel.find(':selected');
    var type = opt.data('type');
    var cfi = opt.data('cfi');
    var strike = opt.data('strike');
    var matDate = opt.data('maturity-date');
    var matMY = opt.data('maturity-my');
    var desc = opt.data('desc');

    this.$('#security_desc').val(desc || '');

    if (type === 'OPT') {
      this.$('#security_type').val('OPT').trigger('change');
      var mmy = matMY || (matDate ? String(matDate).substring(0, 6) : '');
      var day = (matDate && String(matDate).length >= 8) ? String(matDate).substring(6, 8) : '';
      this.$('#maturity_month_year').val(mmy);
      this.$('#maturity_day').val(day ? parseInt(day, 10) : '');
      this.$('#strike_price').val(strike || '');
      if (cfi === 'OC') {
        this.$('#put_or_call').val('1');
      } else if (cfi === 'OP') {
        this.$('#put_or_call').val('0');
      }
    } else if (type === 'FUT') {
      this.$('#security_type').val('FUT').trigger('change');
    } else {
      this.$('#security_type').val('CS').trigger('change');
    }
  },

  updateSecurityType: function() {
    switch(this.$("#security_type option:selected").text()) {
      case "Common Stock":
        this.$("#maturity_month_year").attr({disabled: true, required: false});
        this.$("#maturity_day").attr({disabled: true});
        this.$("#put_or_call").attr({disabled: true, required: false});
        this.$("#strike_price").attr({disabled: true, required: false});
        break;
      case "Future":
        this.$("#maturity_month_year").attr({disabled: false, required: true});
        this.$("#maturity_day").attr({disabled: false});
        this.$("#put_or_call").attr({disabled: true, required: false});
        this.$("#strike_price").attr({disabled: true, required: false});
        break;
      case "Option":
        this.$("#maturity_month_year").attr({disabled: false, required: true});
        this.$("#maturity_day").attr({disabled: false});
        this.$("#put_or_call").attr({disabled: false, required: true});
        this.$("#strike_price").attr({disabled: false, required: true});
        break;
    }
  },

  updateOrdType: function() {
    switch(this.$("#ordType option:selected").text()) {
      case "Limit":
        this.$("#limit").prop("disabled", false);
        this.$("#limit").prop("required", true);
        this.$("#stop").prop("disabled", true);
        this.$("#stop").prop("required", false);
      break;

      case "Stop":
        this.$("#limit").prop("disabled", true);
        this.$("#limit").prop("required", false);
        this.$("#stop").prop("disabled", false);
        this.$("#stop").prop("required", true);
      break;

      case "Stop Limit":
        this.$("#limit").prop("disabled", false);
        this.$("#limit").prop("required", true);
        this.$("#stop").prop("disabled", false);
        this.$("#stop").prop("required", true);
      break;

      default:
        this.$("#limit").prop("disabled", true);
        this.$("#stop").prop("disabled", true);
        this.$("#limit").prop("required", false);
        this.$("#stop").prop("required", false);
    }
  }
});

App.Views.MultilegTicket = Backbone.View.extend({
  template: _.template(`
<h4>Multileg Order</h4>
<form class='form-inline' id='multileg-ticket'>
  <p>
    <div class='form-group'>
      <label for='ml-symbol'>Symbol (root)</label>
      <select class='form-control' name='symbol' id='ml-symbol'>
        <% var underlyings = _.filter(symbols, function(s) { return s.type !== 'OPT'; }); %>
        <% _.each(underlyings, function(s){ %>
          <option value='<%= s.symbol %>'><%= s.description %></option>
        <% }); %>
      </select>
    </div>

    <div class='form-group'>
      <label for='ml-quantity'>Spread Qty</label>
      <input type='number' class='form-control' name='quantity' id='ml-quantity' placeholder='Qty' required>
    </div>

    <div class='form-group'>
      <label for='ml-ordType'>Type</label>
      <select class='form-control' name='ordType' id='ml-ordType'>
        <option value='1'>Market</option>
        <option value='2'>Limit</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='ml-limit'>Limit</label>
      <input type='number' step='.01' class='form-control' id='ml-limit' name='price' placeholder='Limit' disabled>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='ml-account'>Account</label>
      <select class='form-control' name='account' id='ml-account'>
        <% if (!accounts || !accounts.length) { %>
          <option value=''></option>
        <% } %>
        <% _.each(accounts || [], function(a){ %>
          <option value='<%= a %>'><%= a %></option>
        <% }); %>
      </select>
    </div>

    <div class='form-group'>
      <label for='ml-tif'>TIF</label>
      <select class='form-control' name='tif' id='ml-tif'>
        <option value='0'>Day</option>
        <option value='3'>IOC</option>
        <option value='1'>GTC</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='ml-session'>Session</label>
      <select class='form-control' name='session' id='ml-session'>
        <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
      </select>
    </div>
  </p>

  <hr>
  <h5>Legs <button type='button' class='btn btn-success btn-xs add-leg'>+ Add Leg</button></h5>
  <table class='table table-condensed' id='legs-table'>
    <thead>
      <tr>
        <th>Preset</th>
        <th>CFI Code</th>
        <th>Side</th>
        <th>Ratio Qty</th>
        <th>Strike</th>
        <th>Maturity (YYYYMMDD)</th>
        <th>Pos Effect</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
    </tbody>
  </table>

  <button type='submit' class='btn btn-primary'>Submit Multileg Order</button>
</form>
`),

  buildLegPresetOptions: function(root) {
    var allSymbols = this.model.get('symbols') || [];
    var stockSymbol = _.find(allSymbols, function(s) {
      return s.type === 'CS' && s.symbol === root;
    });
    var optSymbols = _.filter(allSymbols, function(s) {
      return s.type === 'OPT' && s.symbol === root;
    });
    var optionsHtml = '<option value="">-- Manual --</option>';
    if (stockSymbol) {
      optionsHtml += '<option value="' + stockSymbol.symbol + ' (Stock)"'
        + ' data-type="CS"'
        + ' data-cfi="ES"'
        + '>' + stockSymbol.symbol + ' (Stock)</option>';
    }
    _.each(optSymbols, function(s) {
      optionsHtml += '<option value="' + s.description + '"'
        + ' data-type="OPT"'
        + ' data-cfi="' + (s.cfi_code || '') + '"'
        + ' data-strike="' + (s.strike_price || '') + '"'
        + ' data-maturity="' + (s.maturity_date || '') + '"'
        + '>' + s.description + '</option>';
    });
    return optionsHtml;
  },

  buildLegRow: function(root) {
    var tpl = _.template(
      '<tr class="leg-row">'
      + '<td><select class="form-control input-sm leg-preset">' + this.buildLegPresetOptions(root) + '</select></td>'
      + '<td><select class="form-control input-sm leg-cfi">'
      +   '<option value="OC">Call</option><option value="OP">Put</option><option value="ES">Stock</option>'
      + '</select></td>'
      + '<td><select class="form-control input-sm leg-side">'
      +   '<option value="1">Buy</option><option value="2">Sell</option>'
      + '</select></td>'
      + '<td><input type="number" class="form-control input-sm leg-ratio" value="1" min="1"></td>'
      + '<td><input type="number" step=".01" class="form-control input-sm leg-strike" placeholder="Strike"></td>'
      + '<td><input type="text" class="form-control input-sm leg-maturity" placeholder="YYYYMMDD"></td>'
      + '<td><select class="form-control input-sm leg-poseffect">'
      +   '<option value="O">Open</option><option value="C">Close</option>'
      + '</select></td>'
      + '<td><button type="button" class="btn btn-danger btn-xs remove-leg">X</button></td>'
      + '</tr>'
    );
    return tpl();
  },

  render: function() {
    this.$el.html(this.template(this.model.attributes));
    this.addLegRow();
    this.addLegRow();
    return this;
  },

  events: {
    "click .add-leg": "addLegRow",
    "click .remove-leg": "removeLegRow",
    "change #ml-symbol": "updateRoot",
    "change #ml-ordType": "updateOrdType",
    "change .leg-cfi": "updateLegFields",
    "change .leg-preset": "updateLegPreset",
    submit: "submit"
  },

  addLegRow: function() {
    var root = this.$('#ml-symbol').val();
    this.$("#legs-table tbody").append(this.buildLegRow(root));
  },

  updateRoot: function() {
    var root = this.$('#ml-symbol').val();
    var optionsHtml = this.buildLegPresetOptions(root);
    this.$('.leg-preset').each(function() {
      $(this).html(optionsHtml).val('');
    });
  },

  removeLegRow: function(e) {
    $(e.target).closest('tr').remove();
  },

  updateOrdType: function() {
    var isLimit = this.$("#ml-ordType").val() === "2";
    this.$("#ml-limit").prop("disabled", !isLimit).prop("required", isLimit);
  },

  updateLegFields: function(e) {
    var row = $(e.target).closest('tr');
    var cfi = $(e.target).val();
    var isOption = (cfi === "OC" || cfi === "OP");
    var isStock = (cfi === "ES");
    row.find('.leg-strike').prop('disabled', !isOption).prop('required', isOption);
    row.find('.leg-maturity').prop('disabled', !isOption).prop('required', isOption);
    if (isStock) {
      row.find('.leg-strike').val('');
      row.find('.leg-maturity').val('');
    }
    row.find('.leg-poseffect').prop('disabled', false);
  },

  updateLegPreset: function(e) {
    var row = $(e.target).closest('tr');
    var opt = $(e.target).find(':selected');
    var presetType = opt.data('type');
    var cfi = opt.data('cfi');
    var strike = opt.data('strike');
    var maturity = opt.data('maturity');

    if (presetType === 'CS') {
      row.find('.leg-cfi').val('ES');
      row.find('.leg-strike').val('').prop('disabled', true).prop('required', false);
      row.find('.leg-maturity').val('').prop('disabled', true).prop('required', false);
      row.find('.leg-poseffect').prop('disabled', false);
      // Default equity leg ratio to 100 shares per contract (covered-call
      // convention). User can override.
      if (row.find('.leg-ratio').val() === '1') {
        row.find('.leg-ratio').val('100');
      }
      return;
    }

    if (cfi) {
      row.find('.leg-cfi').val(cfi);
      row.find('.leg-strike').val(strike || '').prop('disabled', false).prop('required', true);
      row.find('.leg-maturity').val(maturity || '').prop('disabled', false).prop('required', true);
      row.find('.leg-poseffect').prop('disabled', false);
    }
  },

  submit: function(e) {
    e.preventDefault();
    var symbol = this.$('#ml-symbol').val();
    var legs = [];
    this.$('.leg-row').each(function() {
      var row = $(this);
      var cfi = row.find('.leg-cfi').val();
      var isOption = (cfi === "OC" || cfi === "OP");
      legs.push({
        leg_symbol:          symbol,
        leg_cfi_code:        cfi,
        leg_side:            row.find('.leg-side').val(),
        leg_ratio_qty:       parseInt(row.find('.leg-ratio').val()) || 1,
        leg_strike_price:    isOption ? (row.find('.leg-strike').val() || "") : "",
        leg_maturity_date:   isOption ? (row.find('.leg-maturity').val() || "") : "",
        leg_position_effect: row.find('.leg-poseffect').val() || ""
      });
    });

    if (legs.length === 0) {
      alert("Add at least one leg");
      return;
    }

    var order = new App.Models.MultilegOrder();
    order.set({
      symbol:     symbol,
      quantity:   this.$('#ml-quantity').val(),
      ord_type:   this.$('#ml-ordType').val(),
      price:      this.$('#ml-limit').val() || "",
      account:    this.$('#ml-account').val(),
      tif:        this.$('#ml-tif').val(),
      session_id: this.$('#ml-session').val(),
      legs:       legs
    });
    var self = this;
    order.save(null, {
      success: function() {
        self.$('#ml-quantity').val('');
        self.$('#ml-limit').val('');
      },
      error: function(model, response) {
        alert("Error: " + response.responseText);
      }
    });
  }
});

App.prettySide = function(sideEnum) {
  switch(sideEnum) {
    case "1":
      return "Buy";
    case "2":
      return "Sell";
    case "5":
      return "Sell Short";
    case "6":
      return "Sell Short Exempt";
    case "8":
      return "Cross";
    case "9":
      return "Cross Short";
    case "A":
      return "Cross Short Exempt";
  }

  return sideEnum;
};

App.prettyOrdType = function(ordTypeEnum) {
  switch (ordTypeEnum) {
    case "1": return "Market";
    case "2": return "Limit";
    case "3": return "Stop";
    case "4": return "Stop Limit";
  };

  return ordTypeEnum;
};

App.prettySecurityType = function(val) {
  switch (val) {
    case "CS": return "Common Stock";
    case "FUT": return "Future";
    case "OPT": return "Option";
    case "MLEG": return "Multileg";
  }
  return val || "";
};

App.prettyPutOrCall = function(val) {
  switch (String(val)) {
    case "0": return "Put";
    case "1": return "Call";
  }
  return val != null ? val : "";
};

App.prettyOrdStatus = function(val) {
  switch (val) {
    case "0": return "New";
    case "1": return "Partial Fill";
    case "2": return "Filled";
    case "3": return "Done for Day";
    case "4": return "Canceled";
    case "5": return "Replaced";
    case "6": return "Pending Cancel";
    case "7": return "Stopped";
    case "8": return "Rejected";
    case "9": return "Suspended";
    case "A": return "Pending New";
    case "B": return "Calculated";
    case "C": return "Expired";
    case "D": return "Accepted";
    case "E": return "Pending Replace";
  }
  return val || "Pending";
};

App.ordStatusClass = function(val) {
  switch (val) {
    case "0": return "label-info";
    case "1": return "label-primary";
    case "2": return "label-success";
    case "4": return "label-warning";
    case "8": return "label-danger";
    case "A": return "label-default";
    case "E": return "label-default";
  }
  return "label-default";
};




